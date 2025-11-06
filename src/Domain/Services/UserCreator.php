<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Domain\Services;

use App\Domain\Entity\User;
use App\Domain\Repository\UserRepositoryInterface;
use Symfony\Component\DependencyInjection\Attribute\Autowire;
use Symfony\Component\PasswordHasher\Hasher\UserPasswordHasherInterface;

readonly class UserCreator
{
    public function __construct(
        #[Autowire(service: 'app.infrastructure.persistence.doctrine.user.user_repository')]
        private UserRepositoryInterface $userRepository,
        private UserPasswordHasherInterface $userPasswordHasher,
    ){}

    public function create(User $user): int
    {
        $hashPassword = $this->userPasswordHasher->hashPassword($user, $user->getPassword());

        return $this->userRepository->store($user, $hashPassword);
    }

}
