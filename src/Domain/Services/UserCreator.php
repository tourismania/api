<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Domain\Services;

use App\Domain\Entity\User;
use App\Domain\Event\UserRegistered;
use App\Domain\Repository\UserRepositoryInterface;
use Symfony\Component\DependencyInjection\Attribute\Autowire;
use Symfony\Component\Messenger\Exception\ExceptionInterface;
use Symfony\Component\Messenger\MessageBusInterface;
use Symfony\Component\PasswordHasher\Hasher\UserPasswordHasherInterface;

readonly class UserCreator
{
    public function __construct(
        #[Autowire(service: 'app.infrastructure.persistence.doctrine.user.user_repository')]
        private UserRepositoryInterface $userRepository,
        private UserPasswordHasherInterface $userPasswordHasher,
        private MessageBusInterface $messageBus,
    ) {
    }

    /**
     * @throws ExceptionInterface
     */
    public function create(User $user): int
    {
        $hashPassword = $this->userPasswordHasher->hashPassword($user, $user->getPassword() ?? '');

        $id = $this->userRepository->store($user, $hashPassword);

        if ($id === null) {
            throw new \RuntimeException("User save error!");
        }

        $this->messageBus->dispatch(new UserRegistered($id));

        return $id;
    }
}
