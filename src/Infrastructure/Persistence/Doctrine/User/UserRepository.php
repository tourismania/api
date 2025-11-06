<?php

namespace App\Infrastructure\Persistence\Doctrine\User;

use App\Domain\Repository\UserRepositoryInterface;
use DateMalformedStringException;
use Doctrine\Bundle\DoctrineBundle\Repository\ServiceEntityRepository;
use Doctrine\Persistence\ManagerRegistry;
use Symfony\Component\Uid\Uuid;

/**
 * @extends ServiceEntityRepository<User>
 */
class UserRepository extends ServiceEntityRepository implements UserRepositoryInterface
{
    public function __construct(ManagerRegistry $registry)
    {
        parent::__construct($registry, User::class);
    }

    /**
     * @param \App\Domain\Entity\User $user
     * @param string $hashPassword
     *
     * @return int
     *
     * @throws DateMalformedStringException
     * @throws \Random\RandomException
     */
    public function store(\App\Domain\Entity\User $user, string $hashPassword): int
    {
        $userModel = new User();

        $userModel->setFirstName($user->firstName);
        $userModel->setLastName($user->lastName);
        $userModel->setEmail($user->email);
        $userModel->setLogin($user->email);
        $userModel->setUuid(Uuid::v4());
        $userModel->setBirthday(new \DateTime('1994-'.random_int(1, 12).'-'.random_int(1, 31)));
        $userModel->setIsActive(true);
        $userModel->setCreatedAt(new \DateTimeImmutable());
        $userModel->setUpdatedAt(new \DateTimeImmutable());
        $userModel->setPassword($hashPassword);
        $userModel->setPhone('799999999');

        $this->getEntityManager()->persist($userModel);
        $this->getEntityManager()->flush();

        return $userModel->getId();
    }
}
