<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Application\UseCases\CreateUser;

use App\Domain\Entity\User;
use App\Domain\Services\UserCreator;
use Symfony\Component\Messenger\Attribute\AsMessageHandler;

#[AsMessageHandler(bus: 'command.bus')]
readonly class CreateUserCommandHandler
{

    public function __construct(
        private UserCreator $userCreator
    ){}

    public function __invoke(CreateUserCommand $command): CreateUserResult
    {
        $id = $this->userCreator->create(
            new User(
                lastName: $command->lastName,
                firstName:  $command->firstName,
                email: $command->email,
                password: 'qwerty123'
            )
        );

        return new CreateUserResult($id);
    }
}
